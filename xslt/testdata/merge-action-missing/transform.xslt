<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:variable name="doc1" select="doc('testdata/merge-basic/lang1.xml')/lang/language"/>
		<xsl:variable name="doc2" select="doc('testdata/merge-basic/lang2.xml')/lang/language"/>

		<merge-lang>
			<xsl:merge>
				<xsl:merge-source select="$doc1">
					<xsl:merge-key select="@id"/>
				</xsl:merge-source>
				<xsl:merge-source select="$doc2">
					<xsl:merge-key select="@id"/>
				</xsl:merge-source>
			</xsl:merge>
		</merge-lang>
	</xsl:template>
</xsl:stylesheet>