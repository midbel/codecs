<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:variable name="doc1" select="doc('testdata/merge-foreach-item/lang1.xml')"/>
		<xsl:variable name="doc2" select="doc('testdata/merge-foreach-item/lang2.xml')"/>

		<merge-lang>
			<xsl:merge>
				<xsl:merge-source for-each-item="($doc1/lang/language, $doc2/lang/language)" select=".">
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