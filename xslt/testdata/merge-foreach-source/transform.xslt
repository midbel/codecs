<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:variable name="doc1" select="'testdata/merge-foreach-source/lang1.xml'"/>
		<xsl:variable name="doc2" select="'testdata/merge-foreach-source/lang2.xml'"/>

		 <xsl:variable name="sources" select="($doc1, $doc2)"/>

		<merge-lang>
			<xsl:merge>
				<xsl:merge-source for-each-source="$sources"
					select="/lang/language">
					<xsl:merge-key select="@id"/>
				</xsl:merge-source>
				<xsl:merge-action>
					<lang>
						<xsl:value-of select="current-merge-key()"/>
					</lang>
				</xsl:merge-action>
			</xsl:merge>
		</merge-lang>
	</xsl:template>
</xsl:stylesheet>